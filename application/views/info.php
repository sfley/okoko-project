<html>

<head>
<title>info</title>
</head>

<body>


<script src="<?php echo base_url();?>assets/js/highcharts.js"></script>
<script src="<?php echo base_url();?>assets/js/series-label.js"></script>
<script src="<?php echo base_url();?>assets/js/modules/exporting.js"></script>
<script src="<?php echo base_url();?>assets/js/modules/export-data.js"></script>


 <?php header("refresh: 30");?>
 
<!-- <?php var_dump ($info); ?> -->

<br>

<div class="container text-center mb-4">
 
<button type="button" class="btn btn-danger position-relative active mb-1 ">
<?php echo $info['pe1_name']; ?><span class="position-absolute top-0 start-0 translate-middle badge rounded-pill bg-success"><?php echo $info['pe1_temp']; ?>

<span class="visually-hidden"></span>
  </span>
  </button>
    
 <button type="button" class="btn btn-danger active position-relative mb-1"  >
<?php echo $info['pe1_state_text']; ?> 
<span class="badge badge-Dark bg-danger "><?php echo $info['pe1_modulation']; ?></span>
<span class="badge badge-Dark bg-danger "><?php echo $info['pe1_ventilation']; ?></span>
<span class="badge badge-Dark bg-danger "><?php echo $info['pe1_fluegas']; ?></span>
<span class="badge badge-Dark bg-danger "><?php echo $info['pe1_uw_speed']; ?></span>
<span class="badge badge-Dark bg-danger "><?php echo $info['pe1_depression']; ?></span>
  </button>
 
  <br>
  <button type="button" class="btn btn-danger position-relative active mb-1">
  <?php echo $info['pe1_pellets']; ?>
  </button>
  
  <button type="button" class="btn btn-danger position-relative active mb-1">
  <?php echo $info['pe1_pellets_popper']; ?>
    </button>
	
 <button type="button" class="btn btn-danger position-relative active mb-1">
  <?php echo $info['pe1_pellets_today']; ?>
    </button>
  
</div>




 
<div class="container text-center mb-4">
 
<button type="button" class="btn btn-primary position-relative active mb-1 ">
<?php echo $info['hk1_name']; ?><span class="position-absolute top-0 start-0 translate-middle badge rounded-pill bg-success"><?php echo $info['hk1_temp']; ?>

<span class="visually-hidden"></span>
  </span>
  </button>
    
 <button type="button" class="btn btn-primary position-relative mb-1 active ">
<?php echo $info['hk1_state_text']; ?> <span class="badge badge-Dark bg-primary "><?php echo $info['hk1_consigne']; ?></span>
  </button>
 
  
  <button type="button" class="btn btn-primary position-relative mb-1 active">
  Etat de la pompe<span class="position-absolute top-0 start-100 translate-middle badge rounded-pill bg-success"><?php echo $info['hk1_pump']; ?>
    <span class="visually-hidden"></span>
  </span>
  </button>
</div>




<div class="container text-center mb-4">
 
<button type="button" class="btn btn-secondary position-relative mb-1 active ">
<?php echo $info['hk2_name']; ?><span class="position-absolute top-0 start-0 translate-middle badge rounded-pill bg-success"><?php echo $info['hk2_temp']; ?>

<span class="visually-hidden"></span>
  </span>
  </button>
    
 <button type="button" class="btn btn-secondary position-relative mb-1 active ">
<?php echo $info['hk2_state_text']; ?> <span class="badge badge-Dark bg-secondary "><?php echo $info['hk2_consigne']; ?></span>
  </button>
 
  
  <button type="button" class="btn btn-secondary position-relative mb-1 active">
  Etat de la pompe<span class="position-absolute top-0 start-100 translate-middle badge rounded-pill bg-success"><?php echo $info['hk2_pump']; ?>
    <span class="visually-hidden"></span>
  </span>
  </button>
</div>


<div class="container text-center mb-4">

<button type="button" class="btn btn-warning position-relative mb-1 active">
<?php echo $info['ww1_name'];?><span class="position-absolute top-0 start-0 translate-middle badge rounded-pill bg-success"><?php echo $info['ww1_temp'];?>
<span class="visually-hidden"></span>
  </span>
  </button>
  
  <button type="button" class="btn btn-warning position-relative mb-1 active ">
<?php echo $info['ww1_state_text'];?> <span class="badge badge-Dark bg-warning text-dark "><?php echo $info['ww1_consigne'];?></span>
  </button>
  
  <button type="button" class="btn btn-warning position-relative mb-1 active">
  Etat de la pompe<span class="position-absolute top-0 start-100 translate-middle badge rounded-pill bg-success"><?php echo $info['ww1_pump'];?>
    <span class="visually-hidden"></span>
  </span>
  </button>
	  
	  
	  
	  
    </div>


</body>
</html>

